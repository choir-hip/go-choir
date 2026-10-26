# D2 design: memory snapshots that survive a platform deploy (2026-10-26)

Station D (density), item D2 of
[choir-density-2026-10-26.md](../definitions/choir-density-2026-10-26.md).
It supersedes the L20 section of
[fp10-snapshot-levers-and-host-slab-2026-10-22.md](fp10-snapshot-levers-and-host-slab-2026-10-22.md)
(see §4.1 for where and why it differs). Evidence:
[start-refused-for-memory-no-reclaim-2026-10-26.md](../problems/start-refused-for-memory-no-reclaim-2026-10-26.md) §6.3,
[hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md](../problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md) R1.

- **Revision:** revised 2026-10-26 after review 306 (tier 2: Devin,
  "approve with required amendments"; DeepSeek v4.1 flash, "accept the
  shape, do not open S6 as written"). Every finding is answered by name in
  §12.
- **Mutation class of this document:** green. Every build slice is red
  (VM lifecycle, snapshot validation, boot path, deploy activation)
  except S1, which is orange, and S1c, which is yellow (a CI check).
- **Review tier:** this design is reviewed like the change it designs:
  **tier 2** (Devin and DeepSeek v4.1 flash). A defect here can be silent
  and can damage data: loading guest memory against disk bytes it was not
  taken against corrupts a computer without an error. S3 is tier 2. S1,
  S1b, S2, S4 and S5 are additive or fail closed, so they are tier 1. S1c
  is a security gate, so it gets one seat (Devin) even though CI tooling
  is otherwise tier 0.
- **Owner inputs:** "If we can restore fast we can hibernate idle computers
  at will" (2026-10-26). Security first, then availability, then
  customizations (2026-10-25). The floor is "all host-level and
  security-relevant code, everything engineering has no access to"
  (2026-10-25 Q2). The support window is one week (2026-10-25 Q1). Restart
  rules are in CLAUDE.md, "Restarts End Work (Crash) Or Resume It (Planned
  Update)".
- **FP0 ruling (2026-10-26, review 306 DeepSeek finding 6), open to the
  owner's reversal:** under the owner's standing "security comes first"
  ruling and the No Blocking Asks rule, FP0 took the conservative option.
  `security_epoch` is not left to the author's memory. A CI check refuses
  any change to a named security surface outside the floor unless the same
  change raises `security_epoch` or carries an explicit, reviewed "no
  security effect" marker (§6.6, slice S1c). This replaces the old default
  of Q-D2-2.
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
   still serves the guest's ABI; the release is current or inside the
   support window; and the Firecracker version, host kernel, confinement
   and host boot policy are unchanged. Then it resumes onto **its own** release. A
   computer that resumed behind the current release is updated at its next
   idle moment by an ordinary planned, marker-backed refresh ("resume,
   then update"). If any condition fails, the snapshot never resumes and
   the wake cold boots onto the current release, as every wake does today.
3. **Restart rules.** A resume is not a boot, so no marker is consumed or
   written. The update after it is the existing deploy refresh. The guest
   writes its marker only when it is not busy. The refreshed boot consumes
   the marker once. A crash in that boot closes work. Busy computers wait
   in the existing pending set, which survives a vmctl restart.
4. **Security.** Floor and security epoch are fixed when the image is
   built and recorded in the image itself. They are compared against the
   **current** release at every wake. A snapshot from before a floor move or
   an epoch bump cannot load. A hibernated computer therefore never runs
   pre-fix code again; its next wake is a cold boot onto the fix. A
   computer running behind is active, so the deploy that carries the fix
   reaches it like any other active computer. A CI check makes the epoch
   bump hard to forget: a change to a named security surface outside the
   floor must raise the epoch or carry a reviewed "no security effect"
   marker (§6.6). The residual is stated plainly in §6.7: a computer that
   stays busy runs its old code until its next idle moment, exactly as a
   busy computer does after any deploy today.
5. **Gate on S6.** The switch opens only after S5 has landed and the
   deploy-level floor measurement (R9, §10) shows that at least half of the
   NixOS-switch deploys over a week keep floor, epoch and ABI. Below that,
   D2 resumes too little to pay for its red surface, and the built-binary
   floor digest comes first.
6. **Proof.** Hibernate d@e. Wait for a deploy that **changes the guest
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
| **Guest store disk** (erofs: the guest's /nix/store, including the runtime binary `autoputer`, updater, capsule broker, frontend) | `kernel-params` `init=` path in the launch hash; bound-file identity | **Real.** The guest's page cache and the file-backed text of running processes describe old erofs blocks. Pages it evicts are read back from the disk, so loading against new bytes corrupts the guest. It is safe only against the **old** bytes. Today's bound-file identity is sampled at hibernate, through the stable symlink, not at launch, so it misses a switch that lands between launch and hibernate (H6, §11). |
| **Runtime binary** | the same as the store disk (it lives inside it, nix/autoputer-vm.nix:239); no separate host-served runtime exists (the comment at nix/node-b.nix:2989, "vmctl serves it to VMs", is stale: the host pointer is read only by the deploy manifest check, node-b-ci-deploy:550) | **Real**, for the same reason. A self-development layered release lives on data.img, which is inside the snapshot. |
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
| **(c) Never** | n/a | Any of these: the floor digest changed; the security epoch changed; the guest ABI is outside the host's range; the release is not current and older than the support window; the snapshot is older than `VM_SNAPSHOT_MAX_AGE`; the release files are missing or changed; the Firecracker version, host kernel, confinement or host boot policy changed; the CPU fingerprint changed or is unknown (S5); or the snapshot was already consumed or its epoch moved. The wake cold boots onto current: a crash boot, which closes open work (none, since the fence guaranteed nothing ran). |

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
    over the paths listed in `nix/guest-floor-paths.txt`, minus
    `*_test.go` and `testdata/` (tests are not in the binaries). Its hash
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
  - **`flake.nix` and `flake.lock`, whole** (review 306 Devin F2). The
    floor binaries are built by `mkGoService`, `commonGoArgs` and
    `goChoirPackages` (flake.nix:287-293, 454-530), and the image by
    `mkGuestImage` (flake.nix:575-604). A change to a build flag or an
    install step there rewrites floor bytes without touching a Go package,
    and `check-guest-floor-paths` cannot see a missing `.nix` file. The
    whole files are listed, not the two locked revisions and the kernel
    module definition alone, because a partial list is exactly how a
    recipe change slips past. This over-approximates: in the 7 days to
    2026-10-26, 30 commits touched `flake.nix`. The built-binary digest
    (R9) is the later refinement that removes the over-count.

  Today that is 35 Choir packages
  (`GOOS=linux go list -deps` of those commands, rerun 2026-10-26),
  including `internal/capsule`, `internal/updater`, `internal/types` and
  `internal/yaegikernel`. This is the owner's floor, "everything
  engineering has no access to". It over-approximates on purpose: a
  wrongly counted floor change only costs a cold boot. A package is listed
  as its whole directory, so files it embeds (`//go:embed` cannot reach
  outside the package directory) and generated files in it are covered.
- **security_epoch** comes from the tracked file
  `nix/guest-release-policy.json`. It is a monotone integer. It rises in
  the commit that carries a security fix outside the floor (for example in
  the runtime or a desk). It never falls: a revert of a commit that raised
  it keeps the raised value. It works like Android's rollback index and the
  update design's `security_epoch`
  (docs/design/computer-update-architecture-rnd-2026-10-24.md §4.2). Under
  FP0's ruling it is **enforced, not declared**: the CI check in §6.6
  refuses a change to a named security surface that neither raises it nor
  carries a reviewed "no security effect" marker, and refuses any commit
  that lowers it.
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
  `{"release": "<id>", "image_target": "/nix/store/<hash>-go-choir-guest-image", "epoch": N, "launched_at": "..."}`.
  It is written and fsynced before Firecracker is spawned, and rewritten at
  every cold boot. It is written **for every launch, switch on or off**
  (S1b): `release` is empty for a legacy launch, and `image_target` is the
  `readlink` of `VM_GUEST_RELEASE_LINK` taken once at launch. Reattach reads
  it (§4.3); the H6 guard reads it at hibernate (§4.4).
- `SnapshotMeta.GuestRelease string` (`json:"guest_release,omitempty"`):
  copied from the realization's config at hibernate. It is additive, with
  no meta version bump: an old meta reads as legacy.
- `SnapshotMeta.LaunchImageTarget string`
  (`json:"launch_image_target,omitempty"`): the launch record's
  `image_target`, copied at hibernate (S1b). Additive in the same way.
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
  cutover script owns that name: it moves a first non-symlink tree aside
  as a recovery ref and refuses a second (nix/node-b.nix:2936-2947; review
  306 Devin F6 corrected the earlier wording). Releases therefore live
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
- **The activation step is its own snippet, and it cannot fail the
  switch** (review 306 DeepSeek finding 4). The earlier draft put it
  inside `go-choir-guest-image`, which runs under `set -euo pipefail`
  (nix/node-b.nix:2866), so an unguarded `mkdir`, `chmod`, `sync` or `mv`
  on a full disk would have aborted the switch before the symlink swap.
  The step is now a separate snippet, `system.activationScripts.go-choir-guest-release`,
  and `go-choir-guest-image` gains `deps = [ "go-choir-guest-release" ]`
  so the release exists before the link moves. A NixOS activation snippet
  that exits non-zero marks the whole switch failed, so the body runs in a
  subshell that checks every step itself and always exits 0:

  ```sh
  # go-choir-guest-release: materialize ${guestImage} as a release; never fail the switch.
  (
    set +e +u +o pipefail
    src="${guestImage}"; rel=/var/lib/go-choir/guest-releases
    id=$(basename "$src" | cut -c1-32)
    note() { echo "go-choir guest release $id not materialized ($1); snapshots of it will cold boot" >&2; }
    [ -n "$retain" ] || exit 0                              # retain: nix let-binding, see §7 switch
    [ -d "$rel/$id" ] && exit 0
    tmp="$rel/.tmp-$id"
    rm -rf "$tmp" && mkdir -p "$tmp" || { note "mkdir"; rm -rf "$tmp"; exit 0; }
    for f in vmlinux initrd rootfs.ext4 storedisk.erofs kernel-params build.json guest-image-manifest release.json; do
      cp --reflink=always "$src/$f" "$tmp/$f" || { note "reflink $f"; rm -rf "$tmp"; exit 0; }
    done
    printf '{"store_path":"%s","first_current_at":"%s"}\n' "$src" "$(date -u +%FT%TZ)" > "$tmp/installed.json" \
      && chmod 0444 "$tmp"/* && chmod 0555 "$tmp" && sync -f "$tmp/installed.json" \
      && mv -T "$tmp" "$rel/$id" \
      || { note "finish"; chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"; exit 0; }
    exit 0
  ) || true
  ```

  Never use `--reflink=auto`: a silent full copy is GBs per deploy.
  Without a release directory, vmctl launches the current image in legacy
  mode, which is safe. **S2 test:** the snippet, run against a fixture
  with a read-only releases directory and with a source missing one file,
  exits 0, leaves no `.tmp-*` and no release directory, and logs the
  reason.
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
  change, as long as §4.3 is right. A release path is immutable, so its
  identity at hibernate is its identity at launch.
- **The H6 guard, for legacy launches (S1b, switch on or off).**
  `diskIdentity` stats the configured path at hibernate
  (snapshot.go:435-446, sampled in `commitSnapshot` after Firecracker
  stops). For a legacy launch that path is the stable symlink, so after a
  switch it already names the new erofs while the guest's memory describes
  the old one. `commitSnapshot` copies the launch record's `image_target`
  into `meta.LaunchImageTarget`. `validateSnapshot` refuses a legacy meta
  whose `LaunchImageTarget` is set and differs from the `readlink` of
  `VM_GUEST_RELEASE_LINK` now, with the new reason
  `image_moved_since_launch`. A link that cannot be read at wake refuses
  with the same reason (review 313 Devin F4). It runs in the same place as R0 to R6. An
  old meta without the field validates as today. A release-bound meta skips
  it, because its paths cannot move.

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
| R6 | `rel.ID != cur.ID` and `now − rel.first_current_at > VM_GUEST_RELEASE_SUPPORT_WINDOW` (default 168h, owner Q1) | `release_out_of_support` |

- **R6 never ages out the current release** (review 306 Devin F4,
  DeepSeek finding 3). The earlier rule refused snapshots of a release
  that was still current once it had been current for a week, a pure
  availability loss with no security gain, and it also refused snapshots
  of a release that became current again after a host rollback. The window
  still runs from `first_current_at`, not from supersession. That is
  conservative: a release that was current for six days gets one day of
  support once superseded.
- **Legacy metas** (`GuestRelease == ""`) skip R0 to R6 and validate
  exactly as today, plus the H6 guard (§4.4). A legacy snapshot of the
  current image therefore still resumes, and one taken before a deploy
  still cold boots.
- **On success.** `ResumeVMFromSnapshot` (snapshot_lifecycle.go:440) sets
  `snap.GuestRelease`, `snap.CurrentRelease` and
  `snap.Behind = meta.GuestRelease != cur.ID`, and keeps `GuestRelease` on
  the instance config. It rewrites the launch record (§3.2) with the
  meta's release and `LaunchImageTarget` and the resumed epoch, before the
  load, so a vmctl restart after the resume reattaches the computer with
  the release it actually runs (§4.3), and its next snapshot carries the
  original launch target (S1b). `Behind` is a **receipt** of what validation saw,
  not the trigger for the update: `cur` was resolved during validation,
  and a switch can repoint the link between validation and the load
  (review 306 DeepSeek finding 2). Enrollment therefore does not depend on
  it (§5.1 step 2).
- **Unchanged:**
  - the one-shot consume (snapshot.go:429);
  - the credential delivery (a fresh envelope minted by today's vmctl);
  - the fail-closed `resumed` handshake;
  - discarding the snapshot names after a healthy resume.

### 4.6 Releases in use, and retention (S2 script, S3 endpoint)

- **The endpoint.** `GET /internal/vmctl/guest-releases` (internal caller
  only) returns
  `{"retain": bool, "current": "<id>|", "complete": bool, "in_use": [{"id", "running": [vm], "snapshots": [vm]}]}`.
  - `running`: manager instances in a live state with a `GuestRelease`.
  - `snapshots`: unconsumed metas from `listSnapshots()`, from every
    root including the vault, that name a release and are younger than
    `VM_SNAPSHOT_MAX_AGE`.
  - `complete`: false when any snapshot root that is configured (the
    state directory, or the vault when one is set) could not be read.
    `listSnapshots` skips an unreadable root silently today
    (snapshot.go:710-713), so the endpoint uses a variant that reports
    which roots it read (review 306 Devin F5).
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
    window that no running VM uses. Its snapshots can never resume (R6).
  - **Never delete** current, previous, or a release a running VM uses,
    by any rule, including the cap and the support window (review 306
    DeepSeek finding 8: the earlier "delete anyway" rule did not exclude
    running releases, which contradicted M1).
  - **Cap:** `GO_CHOIR_GUEST_RELEASE_KEEP_MAX` (default 8). Above it,
    delete releases that only snapshots use, oldest first.
  - **If vmctl does not answer** (connection error, 5xx), delete
    nothing: without an answer the script cannot know what is running.
    **If it answers `complete: false`**, delete only past-window releases
    that the answer does not list as running. If it answers 404 (an older
    vmctl, which never launches from a release), treat `in_use` as empty.
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
2. **Enroll every release-bound resume** (review 306 DeepSeek finding 2).
   The earlier draft enrolled only when `Behind` was true, but `Behind`
   is decided against the current release resolved during validation. A
   wake that validates just before the switch repoints the link loads the
   old release with `Behind=false`, enrolls nothing, and is caught only if
   that deploy refreshes active computers. Now every resume whose meta
   names a release adds the computer to the **existing pending set**
   (internal/vmctl/deploy_refresh_retry.go:86-155) with:
   - a new field `Kind: "woke_behind"`;
   - a new field `Release` = the release it resumed onto;
   - `Target` = the current release's commit at enrollment. It is only a
     placeholder: the sweep resolves it again **at each pass** (step 3).
     It must not be empty, because the pending file loader drops entries
     with an empty target (deploy_refresh_retry.go:106-108);
   - `LastReason: "resumed on release <id>"`.

   The hook goes into `OwnershipRegistry.resumeFromSnapshot`
   (ownership.go:1670) as a callback set from `cmd/vmctl/main.go`
   next to `SetDeployRefreshPendingPath` (:264), so the registry does not
   import the handler. Enrolling a computer that is not behind costs one
   sweep pass: step 3 forgets it.
3. **Resolve at the sweep, then the user-idle gate.** For a `woke_behind`
   entry, the sweep (`SweepDeployRefreshPending`, :256-288) first calls
   `currentRelease()`. If the computer's instance `GuestRelease` equals the
   current id, the entry is forgotten. Otherwise `Target` is set to the
   current release's `build.json` commit, and the existing "already serves
   target" check (:277-283) runs against it. Only then does the gate
   apply: the sweep skips the entry until
   `now − own.LastActiveAt ≥ VMCTL_BEHIND_REFRESH_IDLE` (default 2m).
   Without the gate, a person who has just woken their computer would get
   a 10 s cold boot within a minute. With D3's 10-minute idle hibernate,
   the 2-minute gate fires first in practice. Entries that are not
   `woke_behind` are unchanged.
4. **A deploy replaces the kind.** `settle` (:134-155) keeps a known
   entry's fields when a refresh is deferred, so as written a
   `woke_behind` entry would keep its kind under a CI deploy. S4 changes
   `settle` to clear `Kind` when the caller is not the retry sweep. When a
   CI deploy refresh of the same computer is deferred as busy, the entry
   then holds the deploy's target and an empty `Kind`. A deploy's refresh
   is therefore never held by the user-idle gate; only the "resume, then
   update" entry is.
5. **Update.** The sweep calls the unchanged `trackedDeployRefresh`.
   - The guest writes its planned-restart marker, or refuses while busy
     (deploy_refresh.go:424-447). `allow_busy` applies only to
     critical_protected and public_platform computers (:215-221), which
     never idle-hibernate.
   - vmctl then kills the computer and cold boots it onto current, and
     the boot consumes the marker once.
   - Busy: the entry stays and is asked again every minute. Refreshed or
     failed: the entry is forgotten (:134-155), so there is no kill loop.
   - "Already serves target": forgotten (:277-283).
   - **Not active** (hibernated again, or stopped): the entry is
     forgotten, because `trackedDeployRefresh` answers "skipped, not
     active" (deploy_refresh.go:417-424) and the sweep forgets skipped
     entries. The pending set is durable across a vmctl restart, **not**
     across a hibernate (review 306 DeepSeek finding 8; the earlier
     wording overstated it). Nothing is lost: the next wake validates the
     snapshot against the then-current release (R0 to R6) and enrolls
     again.
6. **The next deploy.** A behind computer is active, so the next deploy's
   CI refresh (`selectDeployRefresh`, deploy_refresh.go:280-315) reaches
   it like any other active computer, when that deploy refreshes active
   computers. `settle` then forgets or replaces the pending entry.

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
| One marker, one resume | A snapshot resume neither writes nor consumes a marker. The remaining hazard is a marker **captured inside a snapshot**, which has two orderings. (1) Mark, then a lifecycle race lets a hibernate win. (2) The hibernate fence closes first (`prepareHibernate`, hibernate.go:282), and then a mark request writes the marker anyway, because `HandleInternalPlannedRestart` (planned_restart.go:469-536) and `markPlannedRestart` (:317) never consult the fence; the fence gates only `EnterWork` (hibernate.go:129-167) (review 306 Devin F3). In both, `marker.clear` fails against the fenced or paused guest (deploy_refresh.go:457-460), and a resumed guest that crashes within the marker's 15 minutes (planned_restart.go:26) would boot as planned and resume work. Both orderings exist today, without D2. S5 item 1 closes both, and S6 waits for S5. |

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
     D2 adds no new deferral of a fix (the residual is in §6.7). Forcing a fix on a **busy** computer (the
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
3. **An unclassified fix is refused at CI, not bounded after the fact.**
   The earlier draft left the epoch to the author and bounded the
   exposure by the support window and the max age. Review 306 (DeepSeek
   finding 6) named this the one place where the design did not keep "a
   stale snapshot cannot skip a security fix" by construction. Under FP0's
   ruling, the CI check in §6.6 now refuses such a change before it can
   deploy. What remains is a wrong "no security effect" marker, which is a
   reviewed claim, and the busy-computer residual in §6.7.
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

### 6.6 The security-epoch check (S1c; FP0 ruling, open to the owner's reversal)

**The ruling.** FP0 ruled on 2026-10-26, under the owner's standing
"security comes first" ruling and the No Blocking Asks rule, for the
conservative option on review 306 DeepSeek finding 6: a change to a named
security surface outside the floor must, **in the same change**, either
raise `security_epoch` or carry an explicit, reviewed "no security effect"
marker. CI refuses anything else. The owner may reverse this; the reversal
is one edit (drop the CI step) and costs nothing at runtime.

**The surfaces file.** `nix/guest-security-surfaces.txt` (new), one
repo-relative directory prefix per line, `#` comments, grouped with one
line of why per group. It names the code that runs in the guest, outside
the floor, and decides what a guest may do, read or reach:

| Group | Prefixes |
|---|---|
| autoputer, the guest runtime and its HTTP surface | `cmd/autoputer/`, `internal/autoputer/`, `internal/server/`, `internal/apihandler/` |
| the desk runtime | `internal/agentcore/`, `internal/actorruntime/`, `internal/actor/`, `internal/deskworker/`, `cmd/desk-worker/`, `internal/toolregistry/` |
| guest credentials and host-guest authentication | `internal/hostauth/`, `internal/dbcred/`, `internal/keyescrow/` |
| grants and authority | `internal/capfloor/`, `internal/eventtablegrant/`, `internal/owneract/` |
| self-development | `internal/selfdev/`, `internal/selfdevprotocol/` |
| guest egress | `internal/hostfetch/`, `internal/gatewayruntime/` |

These are the packages that `cmd/autoputer` and `cmd/desk-worker` import
and the floor commands do not (`go list -deps`, 2026-10-26), filtered to
the ones with an authority, credential, egress or mutation role. Paths
excluded from matching everywhere: `*_test.go`, `*/testdata/*`, `*.md`.
A new guest package with such a role is added to the file in the commit
that creates it; review asks the question.

`internal/capsule/transaction/` is **not** a surface (review 313 Devin
F1). The floor commands import `internal/capsule`, so the floor list names
that directory, and the floor fileset takes a directory with everything
under it. A change to `transaction/` therefore already moves the floor
digest, and every older release-bound snapshot refuses (R3) without any
epoch raise. Listing it as a surface as well would break rule 5 on the
designed lists themselves.

**The script.** `scripts/check-guest-security-epoch [--base REF] [--head REF]`,
in the style of `scripts/check-store-schema-floor.sh` (same `ensure_base`
fetch-and-retry, same fail-closed rules). It reads the diff like this:

1. `range = base..head`. Commits are `git rev-list --no-merges range`.
   For each commit `c`, the touched paths are
   `git diff-tree --no-commit-id --name-only -r c` against its first
   parent, minus the exclusions.
2. `E(x)` is `.security_epoch` of `nix/guest-release-policy.json` at `x`
   (`git show x:nix/guest-release-policy.json`, parsed with `jq`). A
   missing file at the base counts as 0. A file that does not parse at any
   commit that changed it, or at the head, fails.
3. **Monotone.** For every commit that changed the policy file,
   `E(c) ≥ E(c^)`. A fall fails, with no override: a revert of a raise
   keeps the raised value.
4. **Coverage.** `S` is the commits in the range that touch a surface
   path. If `E(head) > E(base)`, the range passes: the deploy refuses every
   older release-bound snapshot (R4), which is what a raise per commit
   would have done. Otherwise every commit in `S` must carry, in its own
   message, both trailers:

   ```
   Security-Effect: none; <why, at least 20 characters>
   Security-Effect-Reviewed-By: <seat>; <ref>
   ```

   `<seat>` is one of `devin`, `deepseek`, `owner`. `<ref>` matches
   `review [0-9]+` or `owner [0-9]{4}-[0-9]{2}-[0-9]{2}`. A trailer in a
   later commit does not cover an earlier one: "the same change".
5. **Sanity.** The surfaces file must exist, and no surface prefix may
   be equal to, contain, or lie under a path in `nix/guest-floor-paths.txt`
   (a surface is outside the floor by definition; the floor moves by
   itself). Containment counts in both directions, because the floor
   fileset includes subdirectories.
6. **The gate protects itself** (review 313 Devin F2). The surfaces list
   is declared, not derived, so a change that narrows the gate in the same
   range as surface code would otherwise pass. Three rules close that:
   - **Gate files need the trailers, always.** A commit that touches any
     of `nix/guest-security-surfaces.txt`,
     `scripts/check-guest-security-epoch`,
     `scripts/selftest-guest-security-epoch`,
     `.github/scripts/deploy-base-select` (it chooses the base), or whose
     `.github/workflows/ci.yml` diff adds or removes a line containing
     `check-guest-security-epoch` or `plan.outputs.base`, must carry both
     trailers, with `Security-Effect: gate change; <why>`. An epoch raise
     does not excuse it: a raise answers a fix, not a weaker gate.
   - **The list is read at both ends.** Matching uses the **union** of the
     surfaces file at the base and at the head. Removing a prefix
     therefore takes effect only for ranges whose base already lacks it,
     that is, after the removal has deployed under the trailers above.
   - **The script is run from both ends.** CI runs
     `git show <base>:scripts/check-guest-security-epoch` (when the base
     has it) as well as the head's copy, and both must pass. An edit that
     neuters the head's script still meets the base's. The base's copy
     also checks that the head's `ci.yml` still invokes the check in the
     `v1-inventory` job, and fails if it does not.

**How it fails.** Exit 1, and one line per uncovered commit:
`guest-security-epoch: FAIL <sha12> "<subject>" touches <paths>`, then the
two remedies. Before the commit lands on main: amend it with the trailers
after its review, or raise `security_epoch` in it. After it is on main: a
follow-up commit that raises `security_epoch` (it covers the whole range
from the deployed base, rule 4). Trailers cannot be added to a pushed
commit. It also fails, with the cause named, when the base cannot be read
after the fetch retry, when the policy file does not parse, when the
surfaces file is missing, and when a gate file changed without its
trailers (rule 6).

**Where it runs, and which base.**
- **CI:** a step in the `v1-inventory` job (Vocabulary Gates,
  .github/workflows/ci.yml:448-480). `check` needs that job, and the
  staging deploy needs `check` (ci.yml:486, 936), so a failure blocks the
  deploy. The job's checkout takes `fetch-depth: 0`.
  - On a push to main, the base is **the live staging commit** that the
    plan job already selects (ci.yml:93-115), exposed as a new
    `plan.outputs.base`. Not the push's before-SHA: a commit that failed
    the check in one push would otherwise drop out of the next push's
    range and deploy unchecked. With the deployed base, the failure
    repeats until a raise or a revert lands.
  - When the plan has no trustworthy staging commit (it forces every
    lane), the base is the newest first-parent commit older than eight
    days (`git rev-list -1 --first-parent --before=8.days head`): a release
    older than the one-week support window cannot resume (R6), and the
    extra day covers deploy latency.
  - On a pull request, the base is the PR base SHA.
- **Locally:** the lane gate before every push, with
  `--base $(git merge-base origin/main HEAD)`. FP0 adds it to the gate list
  in the worker rules and in CLAUDE.md's cheap checks; this design does
  not edit those files.
- **The deploy explanation** (`deploy-impact-classify`) prints
  "security epoch raised N→M" when the policy file changed, and the CI step
  writes each covered commit's `Security-Effect` trailers to the job
  summary, so a reviewed claim is visible at deploy time.

**What it cannot check, stated plainly.** The script verifies that the
trailers exist and are well formed. It cannot verify that the named seat
actually reviewed the change, or that the claim is true. A wrong marker is
the remaining way for an unflagged fix to reach a resumed computer, and it
is bounded by §6.7. The review prompt for a change that touches a
surface asks the seat to answer "security effect: none, or raise the
epoch" explicitly (S1c adds the line to the code-review prompt template).
Rule 6 protects the gate's own files, but it cannot stop a change to the
CI runner itself, to branch protection, or a push that skips CI; those are
outside the repo's reach, as they are for every other check in
`v1-inventory`.

**Cost, measured** (rerun after review 313 with the final list:
`internal/capsule/transaction/` dropped, `internal/texturewrite/` added).
In the 7 days to 2026-10-26, 223 of 887 non-doc commits on origin/main
(25%) touched these prefixes outside tests, 156 of them in
`internal/agentcore` and 4 in `internal/texturewrite`. The first list gave
220 (that count included `internal/texturewrite/`, since removed: §13 item 1). Each needs the trailers or a raise. Orange
and red changes already get a tier 1 seat, so the trailer rides that
review. A tier 0 change that touches a surface now needs a seat or a
raise. A raise costs one cold boot for every hibernated computer, which is
what every deploy costs today. If raises become routine, D2's ceiling
falls; R9 counts deploys that keep the floor **and** the epoch, so the S6
gate sees it.

### 6.7 The residual: a busy resumed computer runs its old code until its next idle moment

DeepSeek finding 6 names the case, and it stays true after the check:
**a continuously active computer that resumed onto an older release runs
that release's code until its next idle moment.** The checks in R0 to R6
act at wake only. Once a computer is running, nothing in D2 stops it.

How the idle gate bounds it:
- **For the "resume, then update" entry** (§5.1): the sweep runs every
  minute and refreshes the computer at the first pass where its user has
  been away for `VMCTL_BEHIND_REFRESH_IDLE` (2 minutes) **and** the guest
  answers not busy (no running run, no self-development operation, no
  live capsule; `plannedRestartBusy`, planned_restart.go:373).
- **For a deploy that carries a fix**, whether by an epoch raise or a
  floor move: its CI refresh asks every active computer at once, with no
  user-idle gate (§5.1 step 4). A busy computer is deferred and asked again
  every minute, and is refreshed at the first minute it is not busy.
- **No hard time bound exists** for a computer that never goes idle. The
  support window and `VM_SNAPSHOT_MAX_AGE` bound how stale a release may be
  **at wake**, not how long a computer runs after it. This is the same
  exposure every busy computer has after any deploy today; D2 adds the
  population of computers that resumed behind, not a new kind of exposure.
  Forcing a fix into a busy computer ("a forced fix is never contained",
  owner 2026-10-25) is the update architecture's job (U-series), not D2's.
- **The protected classes** (critical_protected, public_platform) take a
  marker-backed refresh while busy (`allow_busy`), but they never
  idle-hibernate, so they never resume behind through D2.

## 7. Slices

Use the rules in the goal file for every item: failure modes first (green,
alone); then tests (yellow, red until the code); then the code; push
`lane/<name>` after each commit, never main; the gates before every push;
both nix evals when `nix/` changed. Order: S0, S1, S1b, S1c, S2, S3, S4,
S5; then S6. S1b and S5 repair defects of today's code, so they land
whether or not S6 ever does. **S6 waits for S5** (review 306 DeepSeek
finding 1; the earlier "S5 may follow S6" contradicted this order).

### S0: problem documentation (green)

- Add a "D2" section to
  `docs/problems/density-station-d-failure-modes-2026-10-26.md` in the D1
  shape (Problem, Design, Failure modes M1..M25 from §8, Protected
  surfaces/evidence/rollback). Commit it alone.
- H1 and H6 (§11) are defects of today's code that can boot a guest
  wrongly or load it against the wrong bytes, with D2 off. They get their
  own problem doc,
  `docs/problems/guest-image-switch-window-kernel-params-and-bound-identity-2026-10-26.md`,
  committed before S1b. FP0 decides whether H2 gets one; S3 repairs it.

### S1: release metadata in the guest image (orange, tier 1)

**Files:**
- `nix/guest-release-policy.json` (new):
  `{"security_epoch":1,"guest_abi":1,"guest_abi_min":1}`, with the commit
  rule for raising each in a sibling `nix/guest-release-policy.md`
  comment, or in the flake comment, because JSON has no comments.
- `nix/guest-floor-paths.txt` (new): one repo-relative path per line,
  including `flake.nix` and `flake.lock` (§3.1).
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
- A flake output, `packages.x86_64-linux.guest-release-json`, that
  evaluates to the release.json of the current commit without building the
  image, so R9 can read the floor digest per deployed commit by `nix eval`
  alone.

**Tests, written first:**
1. The check script fails on a fixture list missing `internal/capsule`,
   fails on a list missing `flake.nix`, and passes on the real list.
2. A flake check, `checks.x86_64-linux.guest-floor-commit-free`. It
   evaluates the guest NixOS config with two different `buildCommit`
   specialArgs and asserts that the kernel, initrd and floor-source store
   paths are equal. This proves the floor digest does not move on a
   commit-only change. **If the initrd differs, take it out of the digest
   and record why.**

**Runtime effect:** none (one extra file in the image directory).
**Rollback:** git revert.

### S1b: the switch window, with D2 off (red, tier 1; review 306 DeepSeek finding 7, Devin F1)

H1 and H6 are real today and do not depend on release directories, so
their repair is split out of S3 and lands regardless of the switch.

**Files:**
- `internal/vmmanager/manager.go`: `applyLaunchDefaults` reads
  `VM_KERNEL_PARAMS_FILE` **at each launch**, through a small
  `m.kernelParams()` that falls back to `m.cfg.KernelParams` when the file
  is unset or unreadable (H1). `expectedLaunchConfig` uses the same
  function, so a wake inside the window rebuilds the new parameters, and
  the launch hash of a guest launched before the switch mismatches.
- `launchFirecracker`: write the launch record (§3.2) for every launch,
  with `image_target` from one `readlink`.
- `snapshot.go`, `snapshot_lifecycle.go`: `SnapshotMeta.LaunchImageTarget`,
  copied at `commitSnapshot`; the `image_moved_since_launch` refusal
  (§4.4).

**Tests, written first:**
1. A launch after the kernel-params file changes uses the new contents
   without a vmctl restart; an unreadable file falls back to the cached
   value.
2. Launch with link target A, repoint to B, hibernate, wake: the wake
   refuses `image_moved_since_launch` and cold boots. Without the repoint
   it resumes.
3. A meta without `launch_image_target` validates exactly as today.
4. Outside the window, `buildFirecrackerConfig` and the launch hash are
   byte-identical to today's (the golden test S3 also uses), so the S1b
   deploy voids no snapshot that today's code would keep.

**Rollback:** git revert. A reverted vmctl ignores the meta field.

### S1c: the security-epoch check (yellow, one seat; §6.6)

**Files:**
- `nix/guest-security-surfaces.txt` (new; §6.6 table).
- `scripts/check-guest-security-epoch` (new).
- `scripts/selftest-guest-security-epoch` (new): builds a throwaway git
  repo in a temp directory and runs the check against it.
- `.github/workflows/ci.yml`: `plan.outputs.base`; the step in
  `v1-inventory` with `fetch-depth: 0`.
- `.github/scripts/deploy-impact-classify`: the "security epoch raised"
  explanation.
- The code-review prompt template: the "security effect" question.

**Tests, written first** (the selftest, each case a fixture commit
range): a surface change with no raise and no trailer fails; the same with
both trailers passes; a trailer with an unknown seat or a malformed ref
fails; a trailer in a later commit does not cover an earlier one; a raise
anywhere in the range passes the range; a fall fails even with trailers; a
`_test.go`-only change passes; an unreadable base fails; an unparsable
policy file fails; a prefix equal to, containing or under a floor path
fails, and the real lists pass; a merge commit is skipped. Gate cases
(rule 6): a commit that removes a prefix and changes code under it in the
same range fails without the gate trailer, because the base's list still
names the prefix; a gate-file change with only an epoch raise fails; a
head script edited to always pass still fails against the base's copy; a
`ci.yml` that drops the step fails.

**Rollback:** remove the CI step. Nothing at runtime depends on it.

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
- a past-window release that a running VM uses is never deleted;
- `complete: false`: only past-window releases not listed as running are
  deleted;
- a symlink or an odd name is never deleted;
- the activation snippet exits 0 and leaves no `.tmp-*` on a read-only
  releases directory and on a source missing a file (§4.1).

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
   exactly as under off. A snapshot of the **current** release whose
   `first_current_at` is older than the window resumes (R6 exemption), and
   so does one of a release that became current again after a rollback.
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
    vault root included; an unreadable vault root gives `complete: false`.

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
- `deploy_refresh_retry.go`: the entry's `Kind` and `Release`,
  `enrollResumed`, the sweep-time resolution of the current release, the
  user-idle gate in `SweepDeployRefreshPending`, and `settle` clearing
  `Kind` for a caller other than the retry sweep.
- `cmd/vmctl/main.go`: wiring and env `VMCTL_BEHIND_REFRESH_IDLE`
  (default `2m`).
- `nix/node-b.nix`: the env line.

**Tests, written first** (`deploy_refresh_behind_test.go`, with the
fixed clock and the fake marker the existing tests use):
1. Every release-bound resume enrolls one entry with a non-empty
   placeholder target. A legacy resume enrolls nothing.
2. **The race:** a resume validated against R1 while R1 was current, with
   the link then moved to R2 before the first sweep: the sweep resolves R2,
   sets the target to R2's commit and, after the gate, refreshes. A resume
   onto the release that is still current is forgotten at the first sweep.
3. The sweep skips the entry while `LastActiveAt` is within the gate and
   refreshes it after (`planned_restart=true`).
4. A busy guest keeps the entry. Refreshed or failed forgets it.
5. "Already serves target" forgets it.
6. A vmctl restart reloads the entry from the pending file with `Kind`
   and `Release`.
7. A CI deploy refresh of the same computer settles and forgets the entry;
   a CI refresh deferred as busy replaces it with an empty `Kind`, and the
   gate then no longer applies.
8. A hibernated computer's entry is forgotten at the next sweep.
9. A non-`woke_behind` entry is unaffected by the gate.

**Rollback:** the switch off stops behind resumes, so no new entries
appear. Existing entries are ordinary deferred refreshes.

### S5: hardening (red, tier 1; before S6)

1. **The marker is never captured in a snapshot**, in either ordering
   (§5.3).
   - **Guest, mark then hibernate:** `prepareHibernate` (hibernate.go:281)
     answers 409 busy (`planned_restart_pending`) while a planned-restart
     marker younger than `plannedRestartMaxAge` exists.
   - **Guest, hibernate then mark** (review 306 Devin F3):
     `HandleInternalPlannedRestart` answers 409 busy
     (`hibernate_fenced`) while the hibernate fence holds, before it
     writes anything, and `markPlannedRestart` refuses the same way. Both
     enter the fence as one work occurrence through a non-waiting variant
     of `enter` (`hibernateFence`, hibernate.go:68-161, guarded by its
     `mu`): fenced means refuse at once; otherwise `inflight` counts the
     mark until the marker write returns, so the fence cannot close while
     a marker is being written. No lock is held across the fsync.
   - **Host:** the idle sweep, make-room and operator hibernate skip a
     computer whose deploy refresh is in flight
     (`deployRefreshState().inFlight`, held across the whole refresh,
     deploy_refresh.go:389-393).
   - **Tests** for all three, including the interleaving "fence closes,
     then a mark arrives" (refused, no marker file) and "a mark is inside,
     then the fence is asked" (the fence answers busy, and after the mark
     returns it answers busy again because a marker exists).
2. **CPU fingerprint** (review 306 DeepSeek finding 5: the earlier form
   was fail-open).
   - `SnapshotMeta.HostCPU` = sha256 of the sorted, de-duplicated set of
     (`vendor_id`, `model name`, `flags`, `microcode`) over **every**
     processor in `/proc/cpuinfo`, so a microcode change on any core moves
     it.
   - Validation refuses `host_cpu_mismatch` when both are set and differ.
   - For a **release-bound** meta, an empty `HostCPU`, or a current
     fingerprint that cannot be read, refuses `host_cpu_unknown`. The
     refusal is at wake, not at hibernate: the result is the same (no load
     on an unknown CPU), and the hibernate still frees the memory. A
     legacy meta with an empty value is allowed, as today, so S5 voids no
     existing snapshot.
   - **Tests** with fake cpuinfo: a change on the last core only; an
     unreadable file at hibernate and at wake.

### S6: switch on and prove (red; FP0 runs the landing loop)

**Gates. S6 does not open until all of these hold:**
1. S1, S1b, S1c, S2, S3, S4 and **S5** have landed and deployed
   (review 306 DeepSeek finding 1).
2. **R9, the deploy-level floor measurement (§10), passes its threshold**:
   over at least 7 days and at least 20 NixOS-switch deploys after S1 and
   S1c landed, at least **50%** of those deploys keep the previous
   deployed release's `floor_digest`, `security_epoch` and `guest_abi`
   (review 306 DeepSeek "worth it?"). Below 50%, S6 waits and the
   built-binary floor digest (§10 R9) is designed first.
3. R7 is read and the cap is set from it (§10).
4. FP0's ruling on the security epoch (§6.6) is live in CI, or the owner
   has reversed it in writing.

Then set `snapshotReleaseRetain = true` in nix/node-b.nix. Land it through
the landing loop. Then run §9 P1 on the **next** NixOS-switch deploy after
a d@e hibernate taken under the switch. A snapshot taken before the switch
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
| M1 | Resume onto a deleted or overwritten release | R1 `release_missing`; bound-file identity (S3 test 5). Retention never deletes a running or young release by any rule, the support-window rule included, and deleting a snapshot's release only cold boots it. |
| M2 | Resume below the floor or the security epoch | R3 and R4, compared at every wake with the current release. S3 test 4. |
| M3 | A behind computer is never refreshed | Every release-bound resume enrolls; the sweep resolves the current release each pass, with a 1-minute sweep and a 2-minute user-idle gate. The entry survives a vmctl restart, not a hibernate; the next wake re-validates and re-enrolls. A computer that never goes idle is the residual in §6.7. The CI refresh at the next deploy also reaches it. Health lists `behind`. |
| M4 | Disk growth from retained releases | Kept: current, previous, young, in use; the cap (8); deletion past the support window. R7 sizes a release before S6. Logged per run. The snapshot budget (`VM_SNAPSHOT_BUDGET_GIB=48`) is separate and unchanged. |
| M5 | Jailer paths | The source is the release path, so the shared copy is stable per release (§4.2). `confinement_mismatch` and the uid checks are kept. S3 test 8. |
| M6 | Snapshot vault | No vault under the jailer (snapshot_lifecycle.go:131); vault metas are counted in `in_use` (S3 test 10). |
| M7 | Credential delivery on resume | Unchanged. An old guest that cannot install the new envelope fails the handshake and cold boots. |
| M8 | A vmctl restart resumes something | Nothing new runs at boot. Reattach reads the launch record. The pending set only asks guests for planned refreshes. |
| M9 | The switch races a launch | Launch resolves the link once and binds the resolved release paths. A release under 2 h old and the previous one are never pruned. |
| M10 | A reattached VM records the wrong release (today's `m.cfg` paths) | §4.3. With no launch record, the next snapshot is legacy, which is safe. S3 test 7. |
| M11 | `kernel-params` goes stale between the NixOS switch and the vmctl restart (H1) | With the switch on, every launch reads `kernel-params` from the resolved release. S3 test 2. |
| M12 | A planned-restart marker is captured in a snapshot, mark-then-fence or fence-then-mark | S5 item 1, both orderings; S6 waits for S5. |
| M13 | Silent host-guest protocol skew | R5 `guest_abi`, plus the fail-closed handshake. The same skew already exists for deferred computers. |
| M14 | Floor list drift: a floor dependency missing from the digest, including the build recipe | `scripts/check-guest-floor-paths` in CI. `flake.nix` and `flake.lock` are listed whole. The `guest-floor-commit-free` flake check. Over-approximation is safe. |
| M15 | CPU features change under a resumed guest | S5 item 2: every core, and an unknown fingerprint refuses a release-bound meta. |
| M16 | Rollback loads wrong bytes | Off: R0 refuses. Revert: the launch hash mismatches (§7 S3 rollback). Neither path loads. |
| M17 | A behind computer is hibernated again before its refresh (idle sweep, make-room) | Allowed: the snapshot names the old release, which stays in use. Each cycle stays bounded by R6 and the max age. |
| M18 | The update restarts a computer in front of its user | The user-idle gate (S4). The CI refresh at deploy time keeps today's semantics for every active computer, unchanged by D2. |
| M19 | A security fix outside the floor ships without an epoch bump | Refused by the CI check (§6.6, S1c) unless a reviewed "no security effect" marker is wrong; then M25. |
| M20 | A hibernate and wake inside the switch-to-restart window load a legacy guest against the new erofs (H6) | S1b: the launch record's `image_target` and `image_moved_since_launch`; kernel parameters read at each launch. Release-bound launches are immune. S1b tests 2 and 4. |
| M21 | A wake validates just before the link moves, loads the old release with `Behind=false`, and is never refreshed | Every release-bound resume enrolls; the sweep resolves the current release itself (§5.1). S4 test 2. |
| M22 | Materializing a release fails the NixOS switch | A separate activation snippet in a subshell that checks every step and always exits 0 (§4.1). S2 test. |
| M23 | R6 refuses snapshots of the current release after a quiet week, or after a rollback makes an old release current again | R6 exempts the current release. S3 test 4. |
| M24 | Retention deletes a vault snapshot's release while the vault is unreadable | The endpoint reports `complete: false`; the script then deletes only past-window releases not running. S2 and S3 tests. |
| M25 | A "no security effect" marker is wrong | A reviewed claim the check cannot verify (§6.6). Bounded as in §6.7: the computer refreshes at its first idle moment (user away 2 minutes, guest not busy); any later deploy's refresh asks again at its first not-busy minute; and a later raise or floor move refuses its snapshot at the next wake. No hard time bound for a computer that is never idle. |

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
- **R9: how often the floor moves. This is the S6 gate.**
  - **Commit-level data:** in the 7 days to 2026-10-26, 159 of 1,135
    non-doc commits on origin/main touched a floor Go package (both
    reviewers reproduced the order: about 154 of 1,145). The file changes
    are dominated by `internal/capsule` (132 file changes, 69 commits) and
    `internal/yaegikernel` (68 file changes, 48 commits). 19 commits
    touched `nix/autoputer-vm.nix`, 2 `flake.lock`, and 30 `flake.nix`.
  - **Deploy-level proxy, read 2026-10-26 (review 306 asked for the
    deploy-level number).** Take the successful CI runs on main from
    2026-10-21 09:35 to 2026-10-26 13:22 (345 runs; the `gh run list`
    limit reached back that far) as the deploy sequence. Classify each
    consecutive pair's diff with `.github/scripts/deploy-impact-classify`:
    121 pairs would run a NixOS switch. Of those:
    - 80 (66%) keep the floor paths without `flake.nix`/`flake.lock`;
    - **74 (61%) keep them with `flake.nix`/`flake.lock` listed** (the
      revised floor);
    - 45 (37%) keep them and touch no §6.6 security surface, which is
      the floor if every surface change were answered with an epoch raise
      instead of a marker.

    It is a proxy and it flatters D2: real deploys batch several CI runs,
    and a larger diff is more likely to move the floor. It says the 50%
    gate is reachable, and that the marker path, not the raise, is what
    keeps D2 above it.
  - **The measurement that gates S6.** After S1 and S1c land, for each
    staging deploy that ran a NixOS switch (the deploy receipt's
    `deploy_host_os=true`), read the deployed commit's release.json with
    `nix eval .#packages.x86_64-linux.guest-release-json` and compare
    `floor_digest`, `security_epoch` and `guest_abi` with the previous
    switch deploy's. Record the series in
    `docs/evidence/density-d2-floor-share-<date>.md`. **Threshold:** over
    at least 7 days and at least 20 switch deploys, at least 50% keep all
    three. That share is D2's ceiling.
  - **Below the threshold**, the refinement comes before S6: compute the
    floor from **built** floor binaries without the commit stamp, rather
    than from source paths. Go builds are reproducible, so only real code
    and recipe changes would move it, and listing `flake.nix` whole would
    no longer be needed. That is a later slice with its own design.

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
  - S1b repairs it with the switch off as well (review 306 DeepSeek
    finding 7: the earlier draft repaired it only under the switch).
- **H2.** `ReattachVMWithConfig` gives a VM running an older image the
  current image's paths (manager.go:1517-1527). Today it is masked by the
  launch hash. S3 repairs it.
- **H3.** Commit stamping makes every NixOS switch a guest change
  (§1.1). It is a substrate cause of snapshot voiding. Removing it would
  touch build identity, so it is out of scope and recorded only.
- **H4.** No snapshot hibernate on staging since 2026-10-24 08:23 (R8).
- **H5.** No CPU template and no CPU fingerprint check (§1.2). S5
  repairs it.
- **H6** (review 306 Devin F1). A silent-corruption path in today's
  legacy snapshots. Between the NixOS switch and the vmctl restart,
  `m.cfg.KernelParams` is stale (H1). A guest launched before the switch,
  hibernated and woken inside that window, passes every check: the launch
  hash is rebuilt from the same stale parameters, so it matches; and
  `diskIdentity` stats the stable symlink at hibernate
  (snapshot.go:435-446) and again at wake, both times seeing the **new**
  erofs, so it matches too. Guest memory then loads against store-disk
  bytes it was never taken against. The comment on `snapshotBoundPaths`
  (snapshot.go:448-454) claims this is guarded; it guards only a
  replacement between hibernate and wake, not between launch and
  hibernate. The code path is certain; whether it has happened is
  unobserved (the window is the seconds-to-minutes between the switch and
  `systemctl restart go-choir-vmctl`, node-b-ci-deploy:508-516). Verified
  here for direct launches; the jailed path was not traced, and S1b's
  guard does not depend on it, because it compares link targets, not file
  identities. S1b repairs it for legacy launches; release-bound launches
  cannot hit it.
- **H7.** The comment at nix/node-b.nix:2989 says vmctl serves the
  autoputer binary to VMs; the guest execs it from its own store disk
  (nix/autoputer-vm.nix:239). A stale comment, recorded only.

**Introduced:** none (a design). **Repaired:** none yet.

**Owner questions**, each with a default (none blocks the build):

- **Q-D2-1.** May a woken computer run the previous runtime for a few
  minutes, until its next idle moment, when the floor and the security
  epoch are unchanged? Default: yes, within the one-week support window.
  The residual for a computer that never goes idle is stated in §6.7.
- **Q-D2-2.** Ruled by FP0 on 2026-10-26, open to the owner's reversal:
  the CI check in §6.6. A change to a named security surface raises the
  epoch or carries a reviewed "no security effect" marker in the same
  change. (The earlier default, "the fix's author remembers", is
  withdrawn.)
- **Q-D2-3.** The user-idle gate before the update restart. Default: 2
  minutes.

## 12. Review 306 answers

Review 306 was tier 2: Devin ("approve with required amendments", six
findings) and DeepSeek v4.1 flash ("accept the shape; do not open S6 as
written", nine findings). Each finding was checked against the code
before it was accepted. None was rejected outright. Where a reviewer
offered two fixes, the one taken is named, and so is the reason the other
was not.

**Devin**

| Finding | Verified against | Answer |
|---|---|---|
| F1, a third silent-corruption window (H6) | `diskIdentity` stats the configured path at commit time (snapshot.go:435-446, `commitSnapshot` after Firecracker stops); `expectedLaunchConfig` refills kernel params from the start-time `m.cfg` (manager.go:1232-1257, config.go:47-49) | **Accepted.** H6 in §11, M20 in §8, guard in §4.4, slice S1b. The alternative, "make release-bound launch the only snapshot-eligible mode", was not taken: it would stop legacy snapshots resuming within a deploy, which works today, and S1b's link-target guard closes the window at less cost. |
| F2, floor digest omits the build recipe | flake.nix:287-293, 454-530, 575-604 build the floor binaries and the image | **Accepted.** `flake.nix` and `flake.lock` are listed whole (§3.1, S1). The built-binary digest stays the later refinement and becomes the first step if R9 falls below its threshold (§10). |
| F3, fence-then-mark captures a marker | `HandleInternalPlannedRestart` (planned_restart.go:469-536) and `markPlannedRestart` (:317) never consult the fence; the fence gates only `EnterWork` (hibernate.go:129-167) | **Accepted.** Both orderings in §5.3; S5 item 1 makes the mark a non-waiting work occurrence that refuses while fenced. The hazard exists today, without D2. |
| F4, R6 refuses after a rollback | `first_current_at` is written once, 0444 (§4.1) | **Accepted**, as an exemption for the current release (§4.5). Rewriting `first_current_at` when a release returns was not taken: the file is immutable on purpose. |
| F5, `in_use` silently misses an unreadable vault | `listSnapshots` skips an unreadable root (snapshot.go:710-713) | **Accepted.** `complete` flag on the endpoint; retention fails closed (§4.6, M24). |
| F6, cutover wording | nix/node-b.nix:2936-2947 | **Accepted** (§4.1). |

**DeepSeek v4.1 flash**

| Finding | Verified against | Answer |
|---|---|---|
| 1, S5/S6 order contradiction | the earlier §7 intro and S5 header | **Accepted.** S6 waits for S5; S5's header says so (§7). |
| 2, `Behind` is a validation-time fact | `trackedDeployRefresh` skips a computer that is not active (deploy_refresh.go:417-424); `deploy-impact-classify` sets `deploy_active_vm_refresh` only for some classes | **Accepted**, by the first fix offered: enroll every release-bound resume and resolve the current release at each sweep (§5.1, M21). The second, comparing after load, was not taken: it is the same comparison done once, and the sweep already repeats it. |
| 3, R6 ages out the current release | as Devin F4 | **Accepted**, exemption (§4.5). Measuring the window from supersession was not taken: the earlier start is the conservative one. |
| 4, "never fails the switch" was false | `set -euo pipefail` at nix/node-b.nix:2866 | **Accepted.** A separate snippet in a checked subshell that always exits 0 (§4.1, M22). |
| 5, CPU fingerprint fail-open | the earlier S5 text | **Accepted**: every core, and an unknown fingerprint refuses a release-bound meta. Changed in one respect: the refusal is at wake, not at hibernate, with the same safety and the memory still freed (S5 item 2). |
| 6, `security_epoch` is a manual declaration | `autoputer` and the desk runtime are outside the floor by design (§3.1) | **Accepted; FP0 ruled** (header, §6.6): a CI check over named surfaces, raise or reviewed marker in the same change. The residual DeepSeek names is stated in §6.7: a busy resumed computer runs pre-fix code until its next idle moment, with no hard bound for one that is never idle. The dissent "make the epoch automatic over all guest code" was not taken: it would reduce D2 to case (a'), and the check is the conservative middle. |
| 7, H1 fix bundled behind the switch | config.go:47-49; node-b-ci-deploy:508-516 | **Accepted.** Slice S1b, switch-independent. |
| 8, "durable" overstated; M1 contradicted §4.6 | deploy_refresh_retry.go:276-288 forgets skipped entries | **Accepted.** §5.1 step 5 rewords; retention never deletes a running release by any rule (§4.6, M1). |
| 9, deliverable shape | goal file docs/definitions/choir-density-2026-10-26.md:331-345 | **Accepted.** The fp10 doc gains a short "L20 revised 2026-10-26" section that points here, so there is one authority and the deliverable the goal file named exists. |
| "Worth it?", gate S6 on the deploy-level number | — | **Accepted.** S6 gate 2 (§7) and the R9 method and threshold (§10). A proxy read today puts the share at 61% with the revised floor, 37% if every surface change took a raise. |
| Risk, embeds and generated files | `//go:embed` cannot leave the package directory | **Answered**: whole package directories are listed (§3.1). |

**Re-review 313** (Devin, one seat: approves; S1 to S5 may be built; S6's
gate is concrete) required two amendments inside §6.6, both made:

| Finding | Answer |
|---|---|
| F1, `internal/capsule/transaction/` collides with the floor | **Accepted.** Dropped from the surfaces; the floor's whole `internal/capsule` directory already covers it. Rule 5 now states containment in both directions (§6.6). |
| F2, the gate is not self-protected | **Accepted.** Rule 6: gate files always need the trailers, an epoch raise does not excuse them; surfaces are matched against the union of base and head lists; the check runs from both the base's and the head's script, and the base's copy verifies the CI step still exists (§6.6, S1c tests). |
| F3, `internal/texturewrite/` is a missing surface | **Superseded** (FP0 ruling 2026-10-26 on the build finding, §13 item 1; review 318 nit): `internal/texturewrite/` is under the floor (`cmd/capsule-broker` imports it), so a change to it already moves `floor_digest` and older release-bound snapshots refuse at R3. It is not a surface, and rule 5 would fail if it were listed. |
| F4, unreadable link at wake | **Accepted.** It refuses `image_moved_since_launch` (§4.4). |
| F5, citation drift | `resumeFromSnapshot` corrected to ownership.go:1670. The other two cite this tree correctly (`mark_guest_boot_contract` at deploy-impact-classify:89, `markPlannedRestart` at planned_restart.go:317), so they stay. |

## 13. Build findings (2026-10-26, S1, S1b, S1c worker)

Written while building S1, S1b and S1c in lane `lane/density-d2-build`. Each
item is a place the design is silent, ambiguous, or wrong against the code,
with what the build did and what FP0 should rule.

1. **`internal/texturewrite/` is under the floor, so it cannot be a security
   surface (blocks the §6.6 list as written).** `cmd/capsule-broker`, one of
   the five floor commands, imports `internal/texturewrite` (`GOOS=linux
   CGO_ENABLED=0 go list -deps ./cmd/capsule-broker | grep texturewrite`,
   2026-10-26). The package is therefore one of the 35 floor packages and is
   listed in `nix/guest-floor-paths.txt` (the S1 selftest requires it).
   Review 313 F3 added it to the surfaces on the premise that "the floor
   commands do not" import it ("`cmd/autoputer` imports it" is true, but so
   does the floor). §6.6 rule 5 forbids a surface prefix that equals, contains
   or lies under a floor path, and the task brief says to exclude anything
   under a floor path. The two cannot both hold. This is the same collision
   review 313 F1 found for `internal/capsule/transaction/`, and the same
   answer applies: a change under a floor path already moves `floor_digest`,
   so every older release-bound snapshot refuses at R3 without an epoch
   raise, and the F3 rationale ("a resumed snapshot must not run code that is
   missing an authority fix") is already met. **Built as:** the surfaces file
   omits `internal/texturewrite/`, with a comment saying why; rule 5 passes on
   the real lists. **FP0 to rule:** accept the omission (recommended), or
   reverse F3's premise in another way (for example carve texturewrite out of
   rule 5, which would double-count a floor package and re-open the F1 logic).
   The cost figure in §6.6 (223 of 887, 4 in texturewrite) falls by at most 4.
2. **S0 had not been done.** The D2 section of the failure-modes doc and the
   H1/H6 problem record did not exist, and Problem Documentation First puts
   them before S1 and S1b. They were written first in this lane
   (`1151978c6`).
3. **`packages.x86_64-linux.guest-release-json` cannot be read by `nix eval`
   alone.** A `writeText` derivation's contents need a build, and the worker
   host is not an x86_64-linux builder. The flake therefore also exposes
   `lib.guestReleaseJson` (the same text as a string), and
   `scripts/test-guest-floor-commit-free` reads that. R9 should read the
   string, not the package.
4. **Where the floor-paths check runs.** The design says "the check job". That
   job (`Go Vet + Test + Build`, name `check`) has no Go and only confirms
   other jobs' results. The script needs `go list`, and `go-vet-build` runs
   only when a Go path changed, so a `flake.nix` or list-only change would skip
   it. It runs in `v1-inventory` (every non-docs change) with a `setup-go`
   step, which `check` also depends on.
5. **The eight-day fallback base is computed in the `v1-inventory` job,** not
   in `plan`: `plan` checks out at depth 2 and cannot run `rev-list --before`.
   `plan.outputs.base` is the deployed commit or empty.
6. **First range after S1c lands.** Staging's commit predates S1c, so the
   first run diffs from it to the S1c head and flags every surface commit in
   between that has no trailers (two of the last 20 main commits, item 13).
   FP0 clears it with one follow-up commit that raises `security_epoch`
   (rule 4), or by amending before landing. A base that lacks the policy file
   counts as epoch 0, so a first-ever policy file with epoch 1 reads as a
   raise; S1 lands and deploys before S1c, so the staging base has the file.
7. **S1c's own commits trip rule 6** (they add the surfaces list, the script,
   its selftest and the `ci.yml` step): a gate-file change needs `Security-Effect:
   gate change; <why>` and a `Security-Effect-Reviewed-By` trailer, and the
   reviewer's `review N` does not exist yet. The worker did not write a
   reviewer trailer it cannot back. **FP0 adds the trailers when landing S1c,**
   after the seat's review. Until then a local run of the check over the lane
   range fails on exactly those commits, which is the check working.
8. **A launch whose launch record cannot be written fails the launch** (S1b),
   as the `fc-config.json` write already does. The design says the record is
   "written and fsynced before Firecracker is spawned" and does not say what
   happens on failure. A failed write after the config write means the state
   directory is unwritable, in which case the launch would fail anyway.
9. **`VM_GUEST_RELEASE_LINK` defaults to `/var/lib/go-choir/guest` in
   `LoadConfigFromEnv`, not in `DefaultManagerConfig`.** Tests and other
   callers that build a `ManagerConfig` by hand get no link, so the S1b guard
   is off for them (a meta without a launch target validates as today). S3's
   env test should assert the same split.
10. **The launch-config golden file was written on macOS.** It pins the
    Firecracker config and launch hash with fake paths and nothing
    host-dependent that was found, but Linux CI is the first place it runs on
    the deploy OS. If it differs there, regenerate with
    `UPDATE_GOLDEN=1 go test ./internal/vmmanager -run TestLaunchConfigGolden`
    from the vmmanager code of the pre-S1b commit `7b831a361`, never from the S1b code.
11. **No code-review prompt template exists in the repo.** The "security effect"
    question is added to the code-review prompt in
    `skills/agentic-consensus/SKILL.md` (Prompt Construction).
12. **S1b jailed path.** The launch record is written before the confinement
    branch of `launchFirecracker`, so jailed and direct launches both write
    it, and the guard compares link targets. The jailed launch itself was not
    exercised in a test (the fixture runs direct); the write is not
    mode-dependent.
13. **Dry run over the last 20 main commits (2026-10-26, origin/main
    `a5bf97c2d`).** History rebuilt in a throwaway clone with the S1 and S1c
    files overlaid on every commit (so the lists, policy, script and `ci.yml`
    exist at both ends of each range), then the check run once per commit
    (`c^..c`): **2 of 20 would fail**, both `internal/agentcore` red commits
    without trailers (`d5b4f495e`, `9771c7680`: the replay-probe scratch and
    the checkpoint-bind fixes). The whole 20-commit range also fails (the same
    two). The other 18 are docs, tests, `checkpointd`, `vmmanager`/`vmctl`
    host code and scripts, none under a surface. The overlay replaces `ci.yml`
    on every commit, so a commit that edited `ci.yml` is not tested for the
    gate rule in this run. A wider sample is the 25% figure of §6.6.

14. **Review 318 (tier 2, Devin and DeepSeek v4.1 flash) and its adjudication.**
    - *Both, HIGH: the gate can be switched off in the same commit as a
      surface change* (a step `if:`, `exit 0` in the run block, a dropped
      `needs:`). **Accepted, the cheap way:** any change to `ci.yml` is a gate
      change and needs the gate-change trailers (§6.6 rule 6); and the head's
      `ci.yml` must still have no step-level `if:` in `v1-inventory`, `check`
      must still need `v1-inventory` and require its success, and
      `deploy-staging` must still require `check`'s. Selftest cases for each.
    - *DeepSeek, MEDIUM-HIGH: H1 not repaired on a cold `ResumeVM` or on
      `RecoverVMWithConfig`.* **Accepted.** `refreshLaunchArtifacts` blanks
      the stored artifacts and kernel parameters (keeping an explicit
      override and the machine shape) and rebinds, in both. The earlier S3
      `rebindLaunchRelease` in `ResumeVM` is this function now. Design §4.2
      listed recover among the cold boots and was right.
    - *DeepSeek, MEDIUM: the base's copy is skipped when the base is not in
      the checkout.* **Accepted.** The CI step fetches the base first and fails
      if it still cannot be read.
    - *DeepSeek, LOW-MEDIUM: the resume's launch-record rewrite only warns.*
      **Accepted.** It is written before the snapshot is consumed; a failure is
      a cold-boot fallback (`launch_record_failed`).
    - *Devin Low, DeepSeek 6: `guest-floor-commit-free` not in CI.* **Accepted:**
      a `guest-floor-commit-free` job (pure Nix evaluation, when the sbom lane's
      inputs changed) that `check` requires.
    - *DeepSeek 6: a configured but unreadable link at launch turned the guard
      off.* **Accepted:** the launch record carries the sentinel
      `!unreadable`, which never equals a real target, so that guest's
      snapshots refuse at wake. A failed record write stopping the launch is now
      tested.
    - *Nit, both: the §6.6 table still listed `internal/texturewrite/`.*
      Removed; the F3 row records the ruling.
    - *Not changed:* the jailed launch path has no test of its own (the record
      is written before the confinement branch, which the direct-launch test
      exercises); legacy metas still validate as before, as designed.
15. **S2: "vmctl does not answer".** §4.6 says delete nothing; the S2 test list
    says "connection refused: only out-of-support is deleted". They disagree.
    Built as the prose (delete nothing, also on 5xx and an unparsable 200): the
    script cannot know what runs, and M1 says a running release is never
    deleted by any rule. `complete:false` deletes only past-window releases
    that the answer does not list as running, as written.
16. **S2: the activation snippet runs only while `snapshotReleaseRetain` is
    on** (the `[ -n "$retain" ]` line of §4.1's sketch became the Nix
    condition), and is a script (`scripts/node-b-guest-release-materialize`)
    the snippet calls, so it is testable. `GO_CHOIR_RELEASE_REFLINK=auto` is a
    test-only escape; the default is `always`.
17. **S3: `ResumeVM` (a stop, then start from the stored config) and
    `RecoverVMWithConfig` also launch from stored config.** The design named
    `BootVM`, `RefreshVM` and recover; a cold `ResumeVM` was not named. Both now
    re-choose artifacts and kernel parameters (`refreshLaunchArtifacts`, review
    318 DeepSeek 2).
18. **S3: `VMInstanceInfo.Behind` is the resume receipt; health recomputes it
    live** from the current release. A `ReattachVMWithConfig` full-process test
    needs `/proc` and was not run on the worker host; its config half is
    `reattachConfig`, tested directly.
19. **S4: the sweep compares the running VM's release (from the manager),
    not the entry's `Release`, with the current release.** An entry for a
    computer that already has a deploy entry is not overwritten. `guestReleaseReader`
    gained `CurrentGuestReleaseInfo`.
20. **S5: the host guard sits in `hibernateVMForDesktopWithReason`,** the one
    function the idle sweep, make-room, pressure reclaim and the operator all
    call, and answers `errHibernateGuestBusy` (a skip, never the kill path). An
    unparsable planned-restart marker counts as pending by its file time.
21. **S5: an unreadable `/proc/cpuinfo` at hibernate leaves `HostCPU` empty,**
    and a release-bound snapshot then refuses at wake (`host_cpu_unknown`); the
    hibernate still frees the memory. Test fixtures stub the reader.
